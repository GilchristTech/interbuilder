package behaviors

import (
  . "gilchrist.tech/interbuilder"
  "fmt"
)


var TaskResolverAssetsInferRoot = TaskResolver {
  Name: "assets-infer",
  Id:   "assets-infer-root",
  MatchBlocks: true,
  TaskPrototype: Task {
    Func: TaskAssetsInferRoot,
  },
}


func ResolveTaskAssetsInfer (spec *Spec) error {
  if spec.GetTaskResolverById("assets-infer-root") == nil {
    spec.AddTaskResolver( &TaskResolverAssetsInferRoot )
  }

  return nil
}


/*
  TaskInferAssetsRoot compares each asset in its Assets array with
  the TaskResolvers in the task. Matching TaskResolvers push
  their tasks into the queue. 
*/
func TaskAssetsInferRoot (spec *Spec, tk *Task) error {
  if tk.Resolver == nil {
    return fmt.Errorf("Task Resolver is nil")
  }

  // Map TaskResolver IDs to TaskResolver references
  //
  var spec_matched_resolvers = make(map[string]*TaskResolver)

  // First match TaskResolvers by Spec and Task, narrowing down
  // the search scope before checking by assets, which is a more
  // expensive operation. This also allows TaskResolver IDs of
  // child specs to take priority over equal IDs in parent specs.
  //
  var last_assets_infer_root *TaskResolver
  for search_spec := spec; search_spec != nil; search_spec = search_spec.Parent {
    assets_infer_root := search_spec.GetTaskResolverById("assets-infer-root")
    if assets_infer_root == nil {
      break
    }

    if last_assets_infer_root == assets_infer_root {
      continue
    }

    last_assets_infer_root = assets_infer_root

    // GetTaskResolverById recurses into parent Specs. If no results
    // were found for more one or more Specs, this loop over
    // parents results in redundant checks. Skip ahead by getting
    // the resolver's Spec, and when this loop iteration exits,
    // it will go into that Spec's parent.
    //
    if assets_infer_root.Spec != nil {
      search_spec = assets_infer_root.Spec
    }

    // Now, we are iterating up the Spec heirarchy, with
    // search_spec equalling only Specs with a assets-infer-root,
    // and only where assets_infer_root is found.

    for infer := assets_infer_root.Children; infer != nil; infer = infer.Next {
      if _, exists := spec_matched_resolvers[infer.Id]; exists {
        continue
      }

      if match, err := infer.Match("assets-infer", spec); err != nil {
        return nil
      } else if match != nil {
        spec_matched_resolvers[infer.Id] = infer
      }
    }
  }

  // Next, using the resolvers matched so far, match those tasks
  // using Assets and for each matching resolver, enqueue one
  // task.
  //
  var num_tasks  = 0
  var num_assets = len(tk.Assets)
  defer tk.Println("Pushed", num_tasks, "tasks from", num_assets, "assets")

  for _, asset := range tk.Assets {
    for resolver_id, resolver := range spec_matched_resolvers {
      matched_resolver, err := resolver.MatchWithAsset(asset)

      if err != nil {
        return err
      } else if matched_resolver == nil {
        continue
      }

      tk.Println("Match, running subtask:", matched_resolver.Id)
      new_subtask := matched_resolver.NewTask()
      if err := new_subtask.Run(spec); err != nil {
        return fmt.Errorf("Error while asset inference is building the task queue: %w", err)
      }

      delete(spec_matched_resolvers, resolver_id)
      num_tasks++
    }
  }

  return tk.ForwardAssets()
}
