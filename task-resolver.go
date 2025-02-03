package interbuilder

import (
  "fmt"
  "net/url"
)


/*
  TaskResolvers are a hierarchical system of matching conditions
  for Tasks, and act as the factories to build them. When a Spec
  is told to search for a Task, it will navigate its own
  TaskResolver tree, and if a match isn't found, it will look
  among its parents. 
*/
type TaskResolver struct {
  Name          string
  Url           url.URL
  Id            string
  TaskPrototype Task

  Spec          *Spec
  History       HistoryEntry

  Next          *TaskResolver
  Children      *TaskResolver

  MatchBlocks   bool
  MatchFunc     TaskMatchFunc

  // The TaskResolver Mask is a Task Mask which if defined, only allows
  // children which have masks inside this one. If a child
  // resolver has mask bits outside of this one's, it should be
  // rejected.
  //
  AcceptMask uint64
}


/*
  Searches this Spec, and recursively through its parents, for a
  TaskResolver with a specific ID. Returns nil if none is found.
*/
func (s *Spec) GetTaskResolverById (id string) *TaskResolver {
  for tr := s.TaskResolvers ; tr != nil ; tr = tr.Next {
    if r := tr.GetTaskResolverById(id); r != nil {
      return r
    }
  }

  if s.Parent == nil {
    return nil
  }

  return s.Parent.GetTaskResolverById(id)
}


/*
  Append a TaskResolver to this Spec, taking priority over
  previously-added and parental resolvers.
*/
func (s *Spec) AddTaskResolver (tr *TaskResolver) {
  var end *TaskResolver
  for end = tr ; end.Next != nil ; end = end.Next {}
  end.Next = s.TaskResolvers
  s.TaskResolvers = tr
}


func (tr *TaskResolver) GetTaskResolverById (id string) *TaskResolver {
  if tr.Id == id {
    return tr
  }

  for child := tr.Children ; child != nil ; child = child.Next {
    if r := child.GetTaskResolverById(id); r != nil {
      return r
    }
  }

  return nil
}


/*
  Returns a pointer to this TaskResolver or a child, whichever has greater
  specificity in the resolver tree. If no matching resolver is found, return
  nil instead.
*/
func (tr *TaskResolver) Match (name string, s *Spec) (*TaskResolver, error) {
  if tr.MatchFunc == nil {
    if tr.Name != name {
      return nil, nil
    }

    if tr.MatchBlocks == false {
      child_match, err := tr.MatchChildren(name, s)
      if err != nil { return nil, err }
      if child_match != nil {
        return child_match, nil
      }
    }

    return tr, nil
  }

  tr_matches, err := tr.MatchFunc(name, s)
  if !tr_matches ||  err != nil {
    return nil, err
  }

  if tr.MatchBlocks == false {
    child_match, err := tr.MatchChildren(name, s)
    if err != nil { return nil, err }
    if child_match != nil {
      return child_match, nil
    }
  }

  return tr, nil
}


func (tr *TaskResolver) MatchChildren (name string, spec *Spec) (*TaskResolver, error) {
  for child := tr.Children ; child != nil ; child = child.Next {
    child_match, err := child.Match(name, spec)
    if err != nil {
      return nil, err
    }
    if child_match != nil {
      return child_match, nil
    }
  }

  return nil, nil
}


func (tr *TaskResolver) NewTask() *Task {
  var task Task = tr.TaskPrototype  // shallow copy

  task.Name       = tr.Name
  task.ResolverId = tr.Id
  task.Resolver   = tr

  return &task
}


func (tr *TaskResolver) GetTask (name string, s *Spec) (*Task, error) {
  resolver, err := tr.Match(name, s)
  if resolver == nil || err != nil {
    return nil, err
  }
  if resolver.TaskPrototype.Func == nil && resolver.TaskPrototype.MapFunc == nil {
    return nil, fmt.Errorf("Task resolver has a nil Func and MapFunc")
  }
  return resolver.NewTask(), nil
}




/*
  Append a TaskResolver to this TaskResolver as a child, taking
  priority over previously-added and sub-resolvers.
*/
func (tr *TaskResolver) AddTaskResolver (add *TaskResolver) error {
  // If the Task resolver's acceptance mask is defined, compare
  // it to the Task Mask of the added resolver, and error if it
  // is rejected.
  //
  if accept_mask := tr.AcceptMask | tr.TaskPrototype.Mask; accept_mask != 0 {
    var test_mask = add.AcceptMask | add.TaskPrototype.Mask
    if test_mask == 0 {
      test_mask = TASK_FIELDS
    }

    if TaskMaskValid(accept_mask, test_mask) == false {
      return fmt.Errorf(
        "Cannot add TaskResolver with id '%s' to '%s', Task mask of %04O not valid within acceptance mask %04O",
        add.Id, tr.Id, test_mask, accept_mask,
      )
    }
  }

  var existing_children = tr.Children

  // Search for the last sibling
  //
  var last_sibling *TaskResolver = add
  for ; last_sibling.Next != nil ; last_sibling = last_sibling.Next {}
  last_sibling.Next = existing_children

  tr.Children = add
  return nil
}


func (s *Spec) GetTask (name string, spec *Spec) (*Task, error) {
  for resolver := s.TaskResolvers ; resolver != nil ; resolver = resolver.Next {
    task, err := resolver.GetTask(name, spec)
    if err != nil {
      return nil, fmt.Errorf("Error getting task in TaskResolver %s: %w", resolver.Id, err)
    }
    if task != nil {
      task.Spec = s
      return task, nil
    }
  }

  if s.Parent == nil {
    return nil, nil
  }

  task, err := s.Parent.GetTask(name, spec)
  if err != nil {
    return nil, err
  }

  if task != nil {
    task.Spec = s
  }
  return task, nil
}


/*
  Match a TaskResolver using an asset, comparing it with the this
  resolver's task prototype, and those of this resolver's
  children. Find the deepest-ish matching TaskResolver.
*/
func (tr *TaskResolver) MatchWithAsset (a *Asset) (*TaskResolver, error) {
  // Check this resolver's TaskPrototype for a match.
  // Guard against a non-match without checking children.
  //
  if this_matches, err := tr.TaskPrototype.MatchAsset(a); err != nil {
    return nil, err
  } else if this_matches == false {
    return nil, nil
  }

  // This resolver, tr, matches.
  // Check children for matches, which take precedence
  //
  if tr.MatchBlocks == false {
    child_match, err := tr.MatchChildrenWithAsset(a)
    if err != nil {
      return nil, nil
    } else if child_match != nil {
      return child_match, err
    }
  }

  // No children match, but this resolver does.
  // Return this resolver.
  //
  return tr, nil
}


func (tr *TaskResolver) MatchChildrenWithAsset (a *Asset) (*TaskResolver, error) {
  for child := tr.Children; child != nil; child = child.Next {
    if child_match, err := child.MatchWithAsset(a); err != nil {
      return nil, err
    } else if child_match != nil {
      return child_match, nil
    }
  }
  return nil, nil
}
