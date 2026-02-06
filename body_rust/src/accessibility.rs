// Author: Enkae (enkae.dev@pm.me)
use anyhow::Result;
use serde::{Deserialize, Serialize};
use windows::{
    Win32::{
        System::Com::{CoCreateInstance, CLSCTX_INPROC_SERVER},
        UI::Accessibility::{
            IUIAutomation, IUIAutomationElement, TreeScope_Children, CUIAutomation,
        },
    },
};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UIElement {
    pub name: String,
    pub control_type: String,
    pub bounding_rectangle: String,
    pub children: Vec<UIElement>,
}

impl UIElement {
    pub fn new(name: String, control_type: String, bounding_rectangle: String) -> Self {
        Self {
            name,
            control_type,
            bounding_rectangle,
            children: Vec::new(),
        }
    }
}

pub fn walk_tree(
    element: &IUIAutomationElement,
    depth: u32,
    max_depth: u32,
) -> Result<UIElement> {
    // Extract current element properties
    let name = get_current_name(element)?;
    let control_type = get_current_control_type(element)?;
    let bounding_rectangle = get_current_bounding_rectangle(element)?;

    let mut ui_element = UIElement::new(name, control_type, bounding_rectangle);

    // Recursively get children if we haven't reached max depth
    if depth < max_depth {
        if let Ok(children) = get_child_elements(element) {
            for child in children {
                match walk_tree(&child, depth + 1, max_depth) {
                    Ok(child_ui_element) => ui_element.children.push(child_ui_element),
                    Err(_) => {
                        // Skip unreadable elements gracefully
                        continue;
                    }
                }
            }
        }
    }

    Ok(ui_element)
}

fn get_current_name(element: &IUIAutomationElement) -> Result<String> {
    unsafe {
        let name_bstr = element.CurrentName()?;
        Ok(name_bstr.to_string())
    }
}

fn get_current_control_type(element: &IUIAutomationElement) -> Result<String> {
    unsafe {
        let control_type_bstr = element.CurrentLocalizedControlType()?;
        Ok(control_type_bstr.to_string())
    }
}

fn get_current_bounding_rectangle(element: &IUIAutomationElement) -> Result<String> {
    unsafe {
        let rect = element.CurrentBoundingRectangle()?;
        Ok(format!(
            "left={},top={},right={},bottom={}",
            rect.left, rect.top, rect.right, rect.bottom
        ))
    }
}

fn get_child_elements(element: &IUIAutomationElement) -> Result<Vec<IUIAutomationElement>> {
    unsafe {
        let automation: IUIAutomation = CoCreateInstance(
            &CUIAutomation,
            None,
            CLSCTX_INPROC_SERVER,
        )?;
        let children_array = element.FindAll(TreeScope_Children, &automation.CreateTrueCondition()?)?;
        
        let mut children = Vec::new();
        let length = children_array.Length()?;
        
        for i in 0..length {
            let child = children_array.GetElement(i)?;
            children.push(child);
        }
        
        Ok(children)
    }
}
